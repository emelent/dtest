using Shop.Notifications.Delivery;

namespace Shop.Notifications.Tests.Delivery;

public class OutboxTests
{
    private readonly Outbox _outbox = new();

    private static Message Mail(string to = "roy@example.com") => new(to, "Your order", "Thanks!");

    [Fact]
    public void TrySendNext_EmptyOutbox_IsFalse() => Assert.False(_outbox.TrySendNext(_ => true));

    [Fact]
    public void TrySendNext_Delivers()
    {
        _outbox.Enqueue(Mail());
        Assert.True(_outbox.TrySendNext(_ => true));
        Assert.Equal(0, _outbox.PendingCount);
    }

    [Fact]
    public void TrySendNext_Failure_Requeues()
    {
        _outbox.Enqueue(Mail());
        Assert.False(_outbox.TrySendNext(_ => false));
        Assert.Equal(1, _outbox.PendingCount);
    }

    [Fact]
    public void ThreeFailures_LandInTheDeadLetters()
    {
        _outbox.Enqueue(Mail());
        for (var i = 0; i < Outbox.MaxAttempts; i++) _outbox.TrySendNext(_ => false);
        Assert.Equal(0, _outbox.PendingCount);
        Assert.Single(_outbox.DeadLetters);
    }

    [Fact]
    public void Messages_AreSentInOrder()
    {
        _outbox.Enqueue(Mail("first@example.com"));
        _outbox.Enqueue(Mail("second@example.com"));

        var sent = new List<string>();
        while (_outbox.TrySendNext(m => { sent.Add(m.To); return true; })) { }

        Assert.Equal(new[] { "first@example.com", "second@example.com" }, sent);
    }
}
