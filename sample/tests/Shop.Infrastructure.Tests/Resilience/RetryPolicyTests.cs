using Shop.Infrastructure.Resilience;

namespace Shop.Infrastructure.Tests.Resilience;

public class RetryPolicyTests
{
    private readonly RetryPolicy _policy = new(maxAttempts: 3, firstDelay: TimeSpan.FromMilliseconds(50));

    [Fact]
    public void Execute_WorkThatSucceeds_RunsOnce() => Assert.Equal(1, _policy.Execute(() => { }));

    [Fact]
    public void Execute_RetriesUntilItSucceeds()
    {
        var calls = 0;
        var attempt = _policy.Execute(() =>
        {
            if (++calls < 3) throw new InvalidOperationException("not yet");
        });
        Assert.Equal(3, attempt);
    }

    [Fact]
    public void Execute_GivesUpAfterTheLastAttempt()
    {
        Assert.Throws<InvalidOperationException>(() => _policy.Execute(() => throw new InvalidOperationException("nope")));
    }

    [Theory]
    [InlineData(1, 0)]
    [InlineData(2, 50)]
    [InlineData(3, 100)]
    [InlineData(4, 200)]
    public void DelayBefore_Doubles(int attempt, int milliseconds)
        => Assert.Equal(TimeSpan.FromMilliseconds(milliseconds), _policy.DelayBefore(attempt));

    [Fact]
    public void Execute_HandsEveryDelayToTheWaiter()
    {
        var waits = new List<TimeSpan>();
        var calls = 0;
        _policy.Execute(() =>
        {
            if (++calls < 3) throw new InvalidOperationException("not yet");
        }, waits.Add);
        Assert.Equal(new[] { TimeSpan.Zero, TimeSpan.FromMilliseconds(50), TimeSpan.FromMilliseconds(100) }, waits);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-1)]
    public void Constructor_RejectsNonPositiveAttempts(int attempts)
        => Assert.Throws<ArgumentOutOfRangeException>(() => new RetryPolicy(attempts, TimeSpan.Zero));
}
