using Shop.Core.Pricing;

namespace Shop.Catalog.Products;

public sealed record Product(string Sku, string Name, string Category, Money Price)
{
    /// <summary>The slug a product is reachable by on the storefront.</summary>
    public string Slug => string.Join('-', Name.ToLowerInvariant().Split(' ', StringSplitOptions.RemoveEmptyEntries));
}
