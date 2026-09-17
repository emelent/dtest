using Shop.Core.Pricing;

namespace Shop.Core.Tests.Pricing;

public class MoneyTests
{
    [Fact]
    public void Add_SameCurrency_SumsAmounts()
    {
        var total = Money.Zar(10).Add(Money.Zar(2.5m));
        Assert.Equal(Money.Zar(12.5m), total);
    }

    [Fact]
    public void Add_DifferentCurrency_Throws()
    {
        var usd = new Money(1, "USD");
        Assert.Throws<InvalidOperationException>(() => Money.Zar(1).Add(usd));
    }

    [Theory]
    [InlineData(1, 10)]
    [InlineData(3, 30)]
    [InlineData(0, 0)]
    public void Times_MultipliesByQuantity(int quantity, decimal expected)
    {
        Assert.Equal(expected, Money.Zar(10).Times(quantity).Amount);
    }

    [Fact]
    public void ToString_UsesTwoDecimals()
    {
        Assert.Equal("ZAR 3.50", Money.Zar(3.5m).ToString());
    }
}
