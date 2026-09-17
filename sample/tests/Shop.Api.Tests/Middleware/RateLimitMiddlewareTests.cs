namespace Shop.Api.Tests.Middleware;

public class RateLimitMiddlewareTests
{
    [Fact]
    public void UnderLimit_Passes() => Assert.True(99 < 100);

    [Fact]
    public void OverLimit_Returns429()
    {
        // Deliberately failing so the Api project has a red node too.
        Assert.Equal(429, 428);
    }
}
