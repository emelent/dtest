namespace Shop.Identity.Passwords;

public static class PasswordPolicy
{
    public const int MinimumLength = 10;

    /// <summary>Every rule a password breaks, in a stable order.</summary>
    public static IEnumerable<string> Violations(string password)
    {
        if (password.Length < MinimumLength) yield return "too short";
        if (!password.Any(char.IsDigit)) yield return "needs a digit";
        if (!password.Any(char.IsUpper)) yield return "needs an upper case letter";
        if (password.Any(char.IsWhiteSpace)) yield return "no whitespace";
    }

    public static bool IsAcceptable(string password) => !Violations(password).Any();
}
