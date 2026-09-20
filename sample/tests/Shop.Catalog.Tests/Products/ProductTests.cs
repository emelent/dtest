using Shop.Catalog.Products;
using Shop.Core.Pricing;

namespace Shop.Catalog.Tests.Products;

public class ProductTests
{
    private static Product Make(string name) => new("A1", name, "tools", Money.Zar(10));

    [Theory]
    [InlineData("Hammer", "hammer")]
    [InlineData("Claw Hammer", "claw-hammer")]
    [InlineData("  Rubber  Mallet ", "rubber-mallet")]
    public void Slug_IsLowerCasedAndHyphenated(string name, string expected)
    {
        Assert.Equal(expected, Make(name).Slug);
    }

    [Fact]
    public void Products_WithTheSameValues_AreEqual()
    {
        Assert.Equal(Make("Hammer"), Make("Hammer"));
    }
}
