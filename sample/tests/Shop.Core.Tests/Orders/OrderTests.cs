using Shop.Core.Orders;
using Shop.Core.Pricing;

namespace Shop.Core.Tests.Orders;

public class OrderTests
{
    private static OrderLine Line(string sku, int qty, decimal price) => new(sku, qty, Money.Zar(price));

    [Fact]
    public void Total_EmptyOrder_IsZero() => Assert.Equal(0m, new Order().Total().Amount);

    [Fact]
    public void Total_SumsLines()
    {
        var order = new Order();
        order.Add(Line("A1", 2, 10));
        order.Add(Line("B2", 1, 5.5m));
        Assert.Equal(25.5m, order.Total().Amount);
    }

    [Fact]
    public async Task Total_LargeOrder_IsQuickEnough()
    {
        var order = new Order();
        for (var i = 0; i < 10_000; i++) order.Add(Line($"S{i}", 1, 1));
        await Task.Delay(1200); // pretend this is expensive so the spinner is visible
        Assert.Equal(10_000m, order.Total().Amount);
    }
}
