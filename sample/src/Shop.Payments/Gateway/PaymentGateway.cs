using Shop.Payments.Cards;

namespace Shop.Payments.Gateway;

public sealed record AuthorisationRequest(string Card, decimal Amount, string Reference);

public sealed record AuthorisationResult(bool Approved, string Code)
{
    public static AuthorisationResult Declined(string code) => new(false, code);
}

/// <summary>A stand-in gateway: it approves anything that looks like a card.</summary>
public sealed class PaymentGateway
{
    private readonly HashSet<string> _seen = new();

    public AuthorisationResult Authorise(AuthorisationRequest request)
    {
        if (request.Amount <= 0) return AuthorisationResult.Declined("invalid-amount");
        if (!CardNumber.IsValid(request.Card)) return AuthorisationResult.Declined("invalid-card");
        // The reference makes a retry idempotent, which is what the caller
        // relies on when it cannot tell whether the first attempt landed.
        if (!_seen.Add(request.Reference)) return new AuthorisationResult(true, "duplicate");
        return new AuthorisationResult(true, "approved");
    }
}
