using Shop.Payments.Cards;
using Shop.Payments.Gateway;

namespace Shop.Payments.Tests.Gateway;

public class PaymentGatewayTests
{
    private const string GoodCard = "4539578763621486";

    private readonly PaymentGateway _gateway = new();

    private static AuthorisationRequest Request(string card = GoodCard, decimal amount = 100m, string reference = "ref-1")
        => new(card, amount, reference);

    [Fact]
    public void Authorise_GoodCard_IsApproved()
    {
        var result = _gateway.Authorise(Request());
        Assert.True(result.Approved);
        Assert.Equal("approved", result.Code);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-5)]
    public void Authorise_NonPositiveAmount_IsDeclined(decimal amount)
    {
        Assert.Equal("invalid-amount", _gateway.Authorise(Request(amount: amount)).Code);
    }

    [Fact]
    public void Authorise_BadCard_IsDeclined()
    {
        Assert.False(_gateway.Authorise(Request(card: "1234567812345678")).Approved);
    }

    [Fact]
    public void Authorise_SameReferenceTwice_IsIdempotent()
    {
        _gateway.Authorise(Request(reference: "ref-2"));
        Assert.Equal("duplicate", _gateway.Authorise(Request(reference: "ref-2")).Code);
    }
}

public class RefundTests
{
    [Fact]
    public void Refund_ReturnsTheFullAmount()
    {
        // Deliberately failing: refunds are not implemented, so the gateway
        // gives back nothing and this project shows a red node.
        var refunded = 0m;
        Console.WriteLine($"refunded {refunded:0.00} of 100.00");
        Assert.Equal(100m, refunded);
    }

    [Fact(Skip = "Partial refunds need the settlement file")]
    public void Refund_Partial_LeavesTheRemainder()
    {
    }
}
