namespace Shop.Search.Indexing;

/// <summary>A toy inverted index: every term must match for a document to hit.</summary>
public sealed class TextIndex
{
    private readonly Dictionary<string, HashSet<string>> _postings = new();

    public int TermCount => _postings.Count;

    public void Add(string id, string text)
    {
        foreach (var term in Tokenise(text))
        {
            if (!_postings.TryGetValue(term, out var ids)) _postings[term] = ids = new HashSet<string>();
            ids.Add(id);
        }
    }

    public IReadOnlyCollection<string> Search(string query)
    {
        var terms = Tokenise(query).ToList();
        if (terms.Count == 0) return Array.Empty<string>();

        IEnumerable<string>? hits = null;
        foreach (var term in terms)
        {
            if (!_postings.TryGetValue(term, out var ids)) return Array.Empty<string>();
            hits = hits is null ? ids : hits.Intersect(ids);
        }
        return hits!.OrderBy(id => id, StringComparer.Ordinal).ToList();
    }

    /// <summary>Lower cases, drops punctuation and splits on whitespace.</summary>
    public static IEnumerable<string> Tokenise(string text) =>
        new string(text.Select(c => char.IsLetterOrDigit(c) ? char.ToLowerInvariant(c) : ' ').ToArray())
            .Split(' ', StringSplitOptions.RemoveEmptyEntries);
}
