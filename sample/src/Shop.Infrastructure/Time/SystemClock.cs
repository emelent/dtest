namespace Shop.Infrastructure.Time;

public interface IClock
{
    DateTimeOffset Now { get; }
}

public sealed class SystemClock : IClock
{
    public DateTimeOffset Now => DateTimeOffset.UtcNow;
}

/// <summary>A clock that only moves when a test moves it.</summary>
public sealed class FrozenClock : IClock
{
    public FrozenClock(DateTimeOffset at) => Now = at;

    public DateTimeOffset Now { get; private set; }

    public void Advance(TimeSpan by) => Now += by;
}
