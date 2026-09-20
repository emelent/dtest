namespace Shop.Notifications.Delivery;

public sealed record Message(string To, string Subject, string Body)
{
    public int Attempts { get; private set; }

    public void Attempted() => Attempts++;
}

/// <summary>A queue that gives up on a message after three attempts.</summary>
public sealed class Outbox
{
    public const int MaxAttempts = 3;

    private readonly Queue<Message> _pending = new();
    private readonly List<Message> _dead = new();

    public int PendingCount => _pending.Count;
    public IReadOnlyList<Message> DeadLetters => _dead;

    public void Enqueue(Message message) => _pending.Enqueue(message);

    /// <summary>Hands the next message to <paramref name="send"/>, requeueing it
    /// on failure until it has burned its attempts.</summary>
    public bool TrySendNext(Func<Message, bool> send)
    {
        if (!_pending.TryDequeue(out var message)) return false;

        message.Attempted();
        if (send(message)) return true;

        if (message.Attempts >= MaxAttempts) _dead.Add(message);
        else _pending.Enqueue(message);
        return false;
    }
}
