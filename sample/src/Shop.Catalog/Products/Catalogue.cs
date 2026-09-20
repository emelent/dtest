namespace Shop.Catalog.Products;

/// <summary>An in-memory catalogue keyed by sku.</summary>
public sealed class Catalogue
{
    // Skus are printed on labels in whatever case the supplier felt like, so
    // they are matched case-insensitively throughout.
    private readonly Dictionary<string, Product> _bySku = new(StringComparer.OrdinalIgnoreCase);

    public int Count => _bySku.Count;

    public void Add(Product product)
    {
        if (!_bySku.TryAdd(product.Sku, product))
            throw new InvalidOperationException($"duplicate sku {product.Sku}");
    }

    public Product? Find(string sku) => _bySku.GetValueOrDefault(sku);

    public IEnumerable<Product> InCategory(string category) =>
        _bySku.Values
            .Where(p => string.Equals(p.Category, category, StringComparison.OrdinalIgnoreCase))
            .OrderBy(p => p.Name, StringComparer.Ordinal);
}
