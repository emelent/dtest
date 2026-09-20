using Shop.Infrastructure.Time;

namespace Shop.Infrastructure.Tests.Time;

public class FrozenClockTests
{
    private static readonly DateTimeOffset Noon = new(2026, 1, 1, 12, 0, 0, TimeSpan.Zero);

    [Fact]
    public void Now_DoesNotMoveOnItsOwn()
    {
        var clock = new FrozenClock(Noon);
        Assert.Equal(clock.Now, clock.Now);
    }

    [Fact]
    public void Advance_MovesTheClock()
    {
        var clock = new FrozenClock(Noon);
        clock.Advance(TimeSpan.FromHours(2));
        Assert.Equal(Noon.AddHours(2), clock.Now);
    }
}

public class SystemClockTests
{
    [Fact]
    public void Now_IsUtc() => Assert.Equal(TimeSpan.Zero, new SystemClock().Now.Offset);
}
