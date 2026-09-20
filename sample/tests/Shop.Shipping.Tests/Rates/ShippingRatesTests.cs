using Shop.Shipping.Parcels;
using Shop.Shipping.Rates;

namespace Shop.Shipping.Tests.Rates;

public class ShippingRatesTests
{
    private static Parcel Weighing(decimal kg) => new(kg, 10, 10, 10);

    [Theory]
    [InlineData(0.5, 55)]
    [InlineData(1, 55)]
    [InlineData(3, 95)]
    [InlineData(20, 160)]
    public void Quote_Local_UsesTheBands(decimal kg, decimal expected)
    {
        Assert.Equal(expected, ShippingRates.Quote(Weighing(kg), Zone.Local));
    }

    [Fact]
    public void Quote_AboveTheTopBand_IsChargedPerKilo()
    {
        Assert.Equal(300m, ShippingRates.Quote(Weighing(25m), Zone.Local));
    }

    [Theory]
    [InlineData(Zone.Local, 55)]
    [InlineData(Zone.National, 77)]
    [InlineData(Zone.CrossBorder, 123.75)]
    public void Quote_ScalesWithTheZone(Zone zone, decimal expected)
    {
        Assert.Equal(expected, ShippingRates.Quote(Weighing(1m), zone));
    }

    [Fact]
    public void Quote_Oversized_Throws()
    {
        Assert.Throws<ArgumentException>(() => ShippingRates.Quote(new Parcel(1m, 200, 10, 10), Zone.Local));
    }

    [Fact(Skip = "Waiting on the courier's weekend tariff")]
    public void Quote_Saturday_CostsMore()
    {
    }
}
