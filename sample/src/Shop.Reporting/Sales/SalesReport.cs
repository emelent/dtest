using Shop.Core.Pricing;

namespace Shop.Reporting.Sales;

public sealed record Sale(DateOnly Date, string Category, Money Amount);

public sealed class SalesReport
{
    private readonly List<Sale> _sales = new();

    public void Record(Sale sale) => _sales.Add(sale);

    public Money Total() => _sales.Aggregate(Money.Zar(0), (acc, s) => acc.Add(s.Amount));

    public Money TotalFor(string category) =>
        _sales.Where(s => s.Category == category).Aggregate(Money.Zar(0), (acc, s) => acc.Add(s.Amount));

    /// <summary>Daily totals, oldest first, with empty days left out.</summary>
    public IReadOnlyList<(DateOnly Date, Money Total)> ByDay() =>
        _sales.GroupBy(s => s.Date)
            .OrderBy(g => g.Key)
            .Select(g => (g.Key, g.Aggregate(Money.Zar(0), (acc, s) => acc.Add(s.Amount))))
            .ToList();

    public string? BestCategory() =>
        _sales.GroupBy(s => s.Category)
            .OrderByDescending(g => g.Sum(s => s.Amount.Amount))
            .ThenBy(g => g.Key, StringComparer.Ordinal)
            .FirstOrDefault()?.Key;
}
