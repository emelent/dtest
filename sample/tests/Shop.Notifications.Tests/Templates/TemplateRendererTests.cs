using Shop.Notifications.Templates;

namespace Shop.Notifications.Tests.Templates;

public class TemplateRendererTests
{
    private static readonly Dictionary<string, string> Values = new()
    {
        ["name"] = "Roy",
        ["order"] = "A1",
    };

    [Fact]
    public void Render_SubstitutesPlaceholders()
        => Assert.Equal("Hi Roy, order A1 shipped.", TemplateRenderer.Render("Hi {name}, order {order} shipped.", Values));

    [Fact]
    public void Render_UnknownPlaceholder_IsLeftInPlace()
        => Assert.Equal("Hi {surname}", TemplateRenderer.Render("Hi {surname}", Values));

    [Fact]
    public void Render_RepeatedPlaceholder_IsSubstitutedEveryTime()
        => Assert.Equal("Roy Roy", TemplateRenderer.Render("{name} {name}", Values));

    [Fact]
    public void Render_NoPlaceholders_IsUnchanged()
        => Assert.Equal("plain text", TemplateRenderer.Render("plain text", Values));

    [Fact]
    public void Placeholders_AreListedOnce()
        => Assert.Equal(new[] { "name", "order" }, TemplateRenderer.Placeholders("{name} {order} {name}"));
}
