using System.Text.RegularExpressions;

namespace Shop.Notifications.Templates;

public static partial class TemplateRenderer
{
    [GeneratedRegex(@"\{(\w+)\}")]
    private static partial Regex Placeholder();

    /// <summary>Substitutes {name} placeholders. An unknown name is left alone
    /// rather than blanked, so a broken template is visible in the mail.</summary>
    public static string Render(string template, IReadOnlyDictionary<string, string> values) =>
        Placeholder().Replace(template, m => values.GetValueOrDefault(m.Groups[1].Value, m.Value));

    public static IEnumerable<string> Placeholders(string template) =>
        Placeholder().Matches(template).Select(m => m.Groups[1].Value).Distinct();
}
