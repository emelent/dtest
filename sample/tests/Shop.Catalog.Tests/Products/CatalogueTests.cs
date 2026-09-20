using Shop.Catalog.Products;
using Shop.Core.Pricing;

namespace Shop.Catalog.Tests.Products;

public class CatalogueTests
{
    private readonly Catalogue _catalogue = new();

    private static Product Product(string sku, string name, string category = "tools") =>
        new(sku, name, category, Money.Zar(10));

    [Fact]
    public void Find_UnknownSku_IsNull() => Assert.Null(_catalogue.Find("nope"));

    [Fact]
    public void Add_ThenFind_ReturnsProduct()
    {
        var hammer = Product("A1", "Hammer");
        _catalogue.Add(hammer);
        Assert.Equal(hammer, _catalogue.Find("A1"));
    }

    [Theory]
    [InlineData("a1")]
    [InlineData("A1")]
    public void Find_IgnoresSkuCase(string sku)
    {
        _catalogue.Add(Product("A1", "Hammer"));
        Assert.NotNull(_catalogue.Find(sku));
    }

    [Fact]
    public void Add_DuplicateSku_Throws()
    {
        _catalogue.Add(Product("A1", "Hammer"));
        Assert.Throws<InvalidOperationException>(() => _catalogue.Add(Product("A1", "Mallet")));
    }

    [Fact]
    public void InCategory_IsSortedByName()
    {
        _catalogue.Add(Product("A1", "Saw"));
        _catalogue.Add(Product("A2", "Hammer"));
        _catalogue.Add(Product("A3", "Ladder", "garden"));
        Assert.Equal(new[] { "Hammer", "Saw" }, _catalogue.InCategory("tools").Select(p => p.Name));
    }

    [Fact]
    public void InCategory_Unknown_IsEmpty() => Assert.Empty(_catalogue.InCategory("nope"));
}
