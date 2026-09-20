namespace Shop.Infrastructure.Resilience;

/// <summary>Retries with a doubling backoff, up to <see cref="MaxAttempts"/>.</summary>
public sealed class RetryPolicy
{
    public RetryPolicy(int maxAttempts, TimeSpan firstDelay)
    {
        if (maxAttempts < 1) throw new ArgumentOutOfRangeException(nameof(maxAttempts));
        MaxAttempts = maxAttempts;
        FirstDelay = firstDelay;
    }

    public int MaxAttempts { get; }
    public TimeSpan FirstDelay { get; }

    public TimeSpan DelayBefore(int attempt) =>
        attempt <= 1 ? TimeSpan.Zero : FirstDelay * Math.Pow(2, attempt - 2);

    /// <summary>Runs <paramref name="work"/> until it stops throwing, then returns
    /// the attempt number that succeeded. Delays are handed to the caller's
    /// <paramref name="wait"/> so tests need not really sleep.</summary>
    public int Execute(Action work, Action<TimeSpan>? wait = null)
    {
        for (var attempt = 1; ; attempt++)
        {
            try
            {
                wait?.Invoke(DelayBefore(attempt));
                work();
                return attempt;
            }
            catch when (attempt < MaxAttempts)
            {
            }
        }
    }
}
