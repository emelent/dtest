using Shop.Core.Orders;
using Shop.Core.Pricing;

namespace Shop.Core.Tests.Orders.Validation;

public class OrderValidatorTests
{
    [Fact]
    public void EmptyOrder_IsReported()
    {
        var errors = OrderValidator.Validate(new Order()).ToList();
        Assert.Contains("order has no lines", errors);
    }

    [Fact]
    public void ValidOrder_HasNoErrors()
    {
        var order = new Order();
        order.Add(new OrderLine("A1", 1, Money.Zar(1)));
        Assert.Empty(OrderValidator.Validate(order));
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-1)]
    public void NonPositiveQuantity_IsReported(int quantity)
    {
        var order = new Order();
        order.Add(new OrderLine("A1", quantity, Money.Zar(1)));
        Assert.Contains("A1: quantity must be positive", OrderValidator.Validate(order));
    }

    [Theory]
    [InlineData("A", true)]
    [InlineData("ABC", true)]
    [InlineData("ABCD", false)]
    [InlineData("", false)]
    public void ShortCoupon_IsReported(string coupon, bool reported)
    {
        var order = new Order { Coupon = coupon };
        order.Add(new OrderLine("A1", 1, Money.Zar(1)));
        Assert.Equal(reported, OrderValidator.Validate(order).Contains("coupon too short"));
    }
}

public class CouponRulesTests
{
    [Fact]
    public void Coupons_AreCaseInsensitive()
    {
        // Deliberately failing: there is no coupon normalisation yet.
        var a = new Order { Coupon = "save10" };
        var b = new Order { Coupon = "SAVE10" };
        Assert.Equal(a.Coupon, b.Coupon);
    }
}
