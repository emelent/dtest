namespace Shop.Payments.Cards;

public static class CardNumber
{
    /// <summary>Luhn check digit validation. Spaces and dashes are ignored.</summary>
    public static bool IsValid(string number)
    {
        var digits = number.Where(char.IsDigit).ToArray();
        if (digits.Length < 12) return false;

        var sum = 0;
        for (var i = 0; i < digits.Length; i++)
        {
            var d = digits[^(i + 1)] - '0';
            if (i % 2 == 1) d = d * 2 > 9 ? d * 2 - 9 : d * 2;
            sum += d;
        }
        return sum % 10 == 0;
    }

    /// <summary>Everything but the last four digits, for receipts and logs.</summary>
    public static string Mask(string number)
    {
        var digits = new string(number.Where(char.IsDigit).ToArray());
        if (digits.Length <= 4) return digits;
        return new string('*', digits.Length - 4) + digits[^4..];
    }

    public static string Brand(string number) => number.FirstOrDefault() switch
    {
        '4' => "visa",
        '5' => "mastercard",
        '3' => "amex",
        _ => "unknown",
    };
}
