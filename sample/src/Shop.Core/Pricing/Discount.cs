namespace Shop.Core.Pricing;

public static class Discount
{
    /// <summary>Applies a percentage discount, rounded to cents.</summary>
    public static Money Percent(Money price, int percent)
    {
        if (percent is < 0 or > 100)
            throw new ArgumentOutOfRangeException(nameof(percent));
        var factor = 1m - percent / 100m;
        return price with { Amount = Math.Round(price.Amount * factor, 2) };
    }

    /// <summary>Buy n, pay for n-1. Quantities below n pay full price.</summary>
    public static Money BuyNGetOneFree(Money unit, int quantity, int n)
    {
        var free = quantity / n;
        return unit.Times(quantity - free);
    }
}
