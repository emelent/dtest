using Shop.Identity.Sessions;

namespace Shop.Identity.Tests.Sessions;

public class SessionStoreTests
{
    private DateTimeOffset _now = new(2026, 1, 1, 9, 0, 0, TimeSpan.Zero);
    private readonly SessionStore _store;

    public SessionStoreTests() => _store = new SessionStore(() => _now);

    [Fact]
    public void Issue_ThenValidate_ReturnsTheSession()
    {
        var session = _store.Issue("roy", TimeSpan.FromHours(1));
        Assert.Equal("roy", _store.Validate(session.Token)?.User);
    }

    [Fact]
    public void Validate_UnknownToken_IsNull() => Assert.Null(_store.Validate("nope"));

    [Fact]
    public void Validate_AfterExpiry_IsNull()
    {
        var session = _store.Issue("roy", TimeSpan.FromMinutes(30));
        _now += TimeSpan.FromMinutes(31);
        Assert.Null(_store.Validate(session.Token));
    }

    [Fact]
    public void Validate_ExpiredToken_IsForgotten()
    {
        var session = _store.Issue("roy", TimeSpan.FromMinutes(30));
        _now += TimeSpan.FromHours(1);
        _store.Validate(session.Token);
        Assert.Equal(0, _store.ActiveCount);
    }

    [Fact]
    public void Revoke_EndsTheSession()
    {
        var session = _store.Issue("roy", TimeSpan.FromHours(1));
        _store.Revoke(session.Token);
        Assert.Null(_store.Validate(session.Token));
    }

    [Fact]
    public void Tokens_AreUnique()
    {
        var tokens = Enumerable.Range(0, 50).Select(_ => _store.Issue("roy", TimeSpan.FromHours(1)).Token);
        Assert.Equal(50, tokens.Distinct().Count());
    }
}
