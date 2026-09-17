using Shop.Core.Inventory;

namespace Shop.Core.Tests.Inventory;

public class StockTests
{
    private readonly Stock _stock = new();

    [Fact]
    public void Level_UnknownSku_IsZero() => Assert.Equal(0, _stock.Level("nope"));

    [Fact]
    public void Receive_AddsToLevel()
    {
        _stock.Receive("A1", 5);
        _stock.Receive("A1", 3);
        Assert.Equal(8, _stock.Level("A1"));
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-4)]
    public void Receive_NonPositive_Throws(int quantity)
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => _stock.Receive("A1", quantity));
    }

    [Fact]
    public void TryReserve_WithStock_Succeeds()
    {
        _stock.Receive("A1", 5);
        Assert.True(_stock.TryReserve("A1", 5));
        Assert.Equal(0, _stock.Level("A1"));
    }

    [Fact]
    public void TryReserve_WithoutStock_Fails()
    {
        Assert.False(_stock.TryReserve("A1", 1));
    }

    [Fact(Skip = "Concurrency support is not implemented yet")]
    public void TryReserve_Concurrent_NeverOversells()
    {
    }
}
