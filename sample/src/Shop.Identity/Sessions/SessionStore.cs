namespace Shop.Identity.Sessions;

public sealed record Session(string Token, string User, DateTimeOffset ExpiresAt);

public sealed class SessionStore
{
    private readonly Dictionary<string, Session> _sessions = new();
    private readonly Func<DateTimeOffset> _now;

    // The clock is injected so tests can expire a session without waiting.
    public SessionStore(Func<DateTimeOffset>? now = null) => _now = now ?? (() => DateTimeOffset.UtcNow);

    public Session Issue(string user, TimeSpan lifetime)
    {
        var session = new Session(Guid.NewGuid().ToString("n"), user, _now() + lifetime);
        _sessions[session.Token] = session;
        return session;
    }

    public Session? Validate(string token)
    {
        if (!_sessions.TryGetValue(token, out var session)) return null;
        if (session.ExpiresAt <= _now())
        {
            _sessions.Remove(token);
            return null;
        }
        return session;
    }

    public void Revoke(string token) => _sessions.Remove(token);

    public int ActiveCount => _sessions.Values.Count(s => s.ExpiresAt > _now());
}
