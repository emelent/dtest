using Shop.Shipping.Parcels;

namespace Shop.Shipping.Rates;

public enum Zone
{
    Local,
    National,
    CrossBorder,
}

public static class ShippingRates
{
    // Bands are cumulative: the first band a parcel fits in wins.
    private static readonly (decimal MaxKg, decimal Price)[] Bands =
    [
        (1m, 55m),
        (5m, 95m),
        (20m, 160m),
    ];

    private const decimal PerKgAboveTopBand = 12m;

    public static decimal Quote(Parcel parcel, Zone zone)
    {
        if (parcel.IsOversized) throw new ArgumentException("parcel is oversized", nameof(parcel));

        var weight = parcel.ChargeableWeightKg;
        var basePrice = PerKgAboveTopBand * weight;
        foreach (var (max, price) in Bands)
        {
            if (weight > max) continue;
            basePrice = price;
            break;
        }

        return Math.Round(basePrice * Multiplier(zone), 2);
    }

    public static decimal Multiplier(Zone zone) => zone switch
    {
        Zone.Local => 1m,
        Zone.National => 1.4m,
        Zone.CrossBorder => 2.25m,
        _ => throw new ArgumentOutOfRangeException(nameof(zone)),
    };
}
