using Shop.Identity.Passwords;

namespace Shop.Identity.Tests.Passwords;

public class PasswordPolicyTests
{
    [Theory]
    [InlineData("Correct1Horse")]
    [InlineData("Battery9Staple")]
    public void IsAcceptable_AcceptsStrongPasswords(string password)
        => Assert.True(PasswordPolicy.IsAcceptable(password));

    [Theory]
    [InlineData("short1A", "too short")]
    [InlineData("nodigitshere", "needs a digit")]
    [InlineData("nouppercase1", "needs an upper case letter")]
    [InlineData("With Space1A", "no whitespace")]
    public void Violations_NameTheBrokenRule(string password, string violation)
        => Assert.Contains(violation, PasswordPolicy.Violations(password));

    [Fact]
    public void Violations_AreReportedInAStableOrder()
    {
        Assert.Equal(
            new[] { "too short", "needs a digit", "needs an upper case letter" },
            PasswordPolicy.Violations("abc"));
    }

    [Fact]
    public void Violations_StrongPassword_IsEmpty() => Assert.Empty(PasswordPolicy.Violations("Correct1Horse"));
}

public class BreachListTests
{
    [Fact]
    public void CommonPasswords_AreRejected()
    {
        // Deliberately failing: there is no breach list yet, so "Password123"
        // sails through every rule.
        Assert.False(PasswordPolicy.IsAcceptable("Password123"));
    }
}
