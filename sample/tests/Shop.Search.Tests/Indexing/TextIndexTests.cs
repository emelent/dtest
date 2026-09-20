using Shop.Search.Indexing;

namespace Shop.Search.Tests.Indexing;

public class TokeniserTests
{
    [Theory]
    [InlineData("Claw Hammer", new[] { "claw", "hammer" })]
    [InlineData("HAMMER", new[] { "hammer" })]
    [InlineData("hammer, 500g!", new[] { "hammer", "500g" })]
    [InlineData("   ", new string[0])]
    public void Tokenise_LowerCasesAndSplits(string text, string[] expected)
        => Assert.Equal(expected, TextIndex.Tokenise(text));
}

public class TextIndexTests
{
    private readonly TextIndex _index = new();

    public TextIndexTests()
    {
        _index.Add("A1", "Claw Hammer");
        _index.Add("A2", "Rubber Mallet");
        _index.Add("A3", "Hammer Drill");
    }

    [Fact]
    public void Search_SingleTerm_FindsEveryDocument()
        => Assert.Equal(new[] { "A1", "A3" }, _index.Search("hammer"));

    [Fact]
    public void Search_IsCaseInsensitive() => Assert.Equal(new[] { "A1", "A3" }, _index.Search("HAMMER"));

    [Fact]
    public void Search_EveryTermMustMatch() => Assert.Equal(new[] { "A1" }, _index.Search("claw hammer"));

    [Fact]
    public void Search_UnknownTerm_IsEmpty() => Assert.Empty(_index.Search("spanner"));

    [Fact]
    public void Search_EmptyQuery_IsEmpty() => Assert.Empty(_index.Search("   "));

    [Fact]
    public void Add_SharesPostingsBetweenDocuments() => Assert.Equal(5, _index.TermCount);

    [Fact(Skip = "Stemming is a nice-to-have")]
    public void Search_MatchesPlurals()
    {
    }
}
