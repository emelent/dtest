using System.Globalization;

namespace Shop.Core.Pricing;

public readonly record struct Money(decimal Amount, string Currency)
{
    public static Money Zar(decimal amount) => new(amount, "ZAR");

    public Money Add(Money other)
    {
        if (other.Currency != Currency)
            throw new InvalidOperationException($"Cannot add {other.Currency} to {Currency}");
        return this with { Amount = Amount + other.Amount };
    }

    public Money Times(int quantity) => this with { Amount = Amount * quantity };

    public override string ToString() => $"{Currency} {Amount.ToString("0.00", CultureInfo.InvariantCulture)}";
}
