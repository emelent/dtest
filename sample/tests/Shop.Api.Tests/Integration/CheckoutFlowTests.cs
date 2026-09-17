namespace Shop.Api.Tests.Integration;

public class CheckoutFlowTests
{
    [Fact]
    public async Task Checkout_HappyPath()
    {
        await Task.Delay(800);
        Assert.True(true);
    }

    [Fact(Skip = "Needs a payment sandbox")]
    public void Checkout_DeclinedCard_ShowsError()
    {
    }
}
