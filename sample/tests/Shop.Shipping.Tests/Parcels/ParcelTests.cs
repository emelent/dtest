using Shop.Shipping.Parcels;

namespace Shop.Shipping.Tests.Parcels;

public class ParcelTests
{
    [Fact]
    public void VolumetricWeight_IsVolumeOverFiveThousand()
    {
        var parcel = new Parcel(1m, 50, 40, 30);
        Assert.Equal(12m, parcel.VolumetricWeightKg);
    }

    [Fact]
    public void ChargeableWeight_TakesTheHeavierOfTheTwo()
    {
        var light = new Parcel(1m, 50, 40, 30);
        var heavy = new Parcel(20m, 10, 10, 10);
        Assert.Equal(12m, light.ChargeableWeightKg);
        Assert.Equal(20m, heavy.ChargeableWeightKg);
    }

    [Theory]
    [InlineData(130, 10, 10, true)]
    [InlineData(10, 130, 10, true)]
    [InlineData(10, 10, 130, true)]
    [InlineData(120, 120, 120, false)]
    public void IsOversized_AtAHundredAndTwentyCentimetres(int l, int w, int h, bool oversized)
    {
        Assert.Equal(oversized, new Parcel(1m, l, w, h).IsOversized);
    }
}
