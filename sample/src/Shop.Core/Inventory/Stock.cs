namespace Shop.Core.Inventory;

public sealed class Stock
{
    private readonly Dictionary<string, int> _levels = new();

    public int Level(string sku) => _levels.GetValueOrDefault(sku);

    public void Receive(string sku, int quantity)
    {
        if (quantity <= 0) throw new ArgumentOutOfRangeException(nameof(quantity));
        _levels[sku] = Level(sku) + quantity;
    }

    public bool TryReserve(string sku, int quantity)
    {
        if (Level(sku) < quantity) return false;
        _levels[sku] -= quantity;
        return true;
    }
}
