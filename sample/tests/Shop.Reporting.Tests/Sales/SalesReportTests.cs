using Shop.Core.Pricing;
using Shop.Reporting.Sales;

namespace Shop.Reporting.Tests.Sales;

public class SalesReportTests
{
    private static readonly DateOnly Monday = new(2026, 1, 5);

    private readonly SalesReport _report = new();

    private void Record(DateOnly date, string category, decimal amount)
        => _report.Record(new Sale(date, category, Money.Zar(amount)));

    [Fact]
    public void Total_EmptyReport_IsZero() => Assert.Equal(0m, _report.Total().Amount);

    [Fact]
    public void Total_SumsEverySale()
    {
        Record(Monday, "tools", 100);
        Record(Monday, "garden", 50);
        Assert.Equal(150m, _report.Total().Amount);
    }

    [Fact]
    public void TotalFor_IsPerCategory()
    {
        Record(Monday, "tools", 100);
        Record(Monday, "garden", 50);
        Assert.Equal(100m, _report.TotalFor("tools").Amount);
    }

    [Fact]
    public void ByDay_IsOldestFirst()
    {
        Record(Monday.AddDays(1), "tools", 20);
        Record(Monday, "tools", 10);
        Assert.Equal(new[] { Monday, Monday.AddDays(1) }, _report.ByDay().Select(d => d.Date));
    }

    [Fact]
    public void ByDay_LeavesOutEmptyDays()
    {
        Record(Monday, "tools", 10);
        Record(Monday.AddDays(3), "tools", 10);
        Assert.Equal(2, _report.ByDay().Count);
    }

    [Fact]
    public void BestCategory_IsTheBiggestSeller()
    {
        Record(Monday, "tools", 100);
        Record(Monday, "garden", 250);
        Assert.Equal("garden", _report.BestCategory());
    }

    [Fact]
    public void BestCategory_EmptyReport_IsNull() => Assert.Null(_report.BestCategory());
}

public class MonthEndTests
{
    [Fact]
    public async Task Rollup_OverAYearOfSales()
    {
        var report = new SalesReport();
        var start = new DateOnly(2025, 1, 1);
        for (var i = 0; i < 365; i++) report.Record(new Sale(start.AddDays(i), "tools", Money.Zar(10)));

        await Task.Delay(700); // pretend the warehouse is slow, so the spinner has something to do
        Assert.Equal(3650m, report.Total().Amount);
    }
}
