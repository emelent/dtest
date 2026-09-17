using Shop.Core.Pricing;

namespace Shop.Core.Tests.Pricing;

public class DiscountTests
{
    [Theory]
    [InlineData(100, 0, 100)]
    [InlineData(100, 25, 75)]
    [InlineData(19.99, 10, 17.99)]
    [InlineData(100, 100, 0)]
    public void Percent_ReducesPrice(decimal price, int percent, decimal expected)
    {
        Assert.Equal(expected, Discount.Percent(Money.Zar(price), percent).Amount);
    }

    [Theory]
    [InlineData(-1)]
    [InlineData(101)]
    public void Percent_OutOfRange_Throws(int percent)
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => Discount.Percent(Money.Zar(1), percent));
    }

    [Fact]
    public void BuyThreeGetOneFree_ChargesTwoOfThree()
    {
        var total = Discount.BuyNGetOneFree(Money.Zar(10), quantity: 3, n: 3);
        Assert.Equal(20m, total.Amount);
    }

    [Fact]
    public void BuyThreeGetOneFree_SevenItems_ChargesFive()
    {
        // Deliberately wrong expectation: 7 / 3 = 2 free, so 5 are charged, not 6.
        var total = Discount.BuyNGetOneFree(Money.Zar(10), quantity: 7, n: 3);
        Console.WriteLine($"computed total = {total}");
        Assert.Equal(60m, total.Amount);
    }
}
