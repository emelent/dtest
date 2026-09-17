using Shop.Core.Pricing;

namespace Shop.Core.Orders;

public sealed record OrderLine(string Sku, int Quantity, Money UnitPrice)
{
    public Money Total => UnitPrice.Times(Quantity);
}

public sealed class Order
{
    private readonly List<OrderLine> _lines = new();

    public IReadOnlyList<OrderLine> Lines => _lines;
    public string? Coupon { get; set; }

    public void Add(OrderLine line) => _lines.Add(line);

    public Money Total()
    {
        var total = Money.Zar(0);
        foreach (var line in _lines) total = total.Add(line.Total);
        return total;
    }
}

public static class OrderValidator
{
    public static IEnumerable<string> Validate(Order order)
    {
        if (order.Lines.Count == 0) yield return "order has no lines";
        foreach (var line in order.Lines)
        {
            if (line.Quantity <= 0) yield return $"{line.Sku}: quantity must be positive";
            if (string.IsNullOrWhiteSpace(line.Sku)) yield return "line has no sku";
        }
        if (order.Coupon is { Length: > 0 and < 4 }) yield return "coupon too short";
    }
}
