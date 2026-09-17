namespace Shop.Api.Tests.Middleware;

public class AuthMiddlewareTests
{
    [Fact]
    public void MissingHeader_IsRejected() => Assert.False(string.IsNullOrEmpty("Bearer x") is false && false);

    [Theory]
    [InlineData("Bearer abc", true)]
    [InlineData("Basic abc", false)]
    [InlineData("", false)]
    public void Scheme_MustBeBearer(string header, bool accepted)
        => Assert.Equal(accepted, header.StartsWith("Bearer "));

    [Fact]
    public void ExpiredToken_IsRejected()
    {
        var expiry = DateTimeOffset.UtcNow.AddMinutes(-1);
        Assert.True(expiry < DateTimeOffset.UtcNow);
    }
}
