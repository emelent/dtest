namespace Shop.Shipping.Parcels;

public sealed record Parcel(decimal WeightKg, int LengthCm, int WidthCm, int HeightCm)
{
    /// <summary>Couriers bill the greater of actual and volumetric weight.</summary>
    public decimal VolumetricWeightKg => Math.Round(LengthCm * WidthCm * HeightCm / 5000m, 2);

    public decimal ChargeableWeightKg => Math.Max(WeightKg, VolumetricWeightKg);

    public bool IsOversized => LengthCm > 120 || WidthCm > 120 || HeightCm > 120;
}
