using Shop.Payments.Cards;

namespace Shop.Payments.Tests.Cards;

public class CardNumberTests
{
    [Theory]
    [InlineData("4539578763621486")]
    [InlineData("4539 5787 6362 1486")]
    [InlineData("4539-5787-6362-1486")]
    [InlineData("6011111111111117")]
    public void IsValid_AcceptsLuhnNumbers(string number) => Assert.True(CardNumber.IsValid(number));

    [Theory]
    [InlineData("4539578763621487")]
    [InlineData("1234567812345678")]
    [InlineData("41111")]
    [InlineData("")]
    public void IsValid_RejectsEverythingElse(string number) => Assert.False(CardNumber.IsValid(number));

    [Theory]
    [InlineData("4539578763621486", "************1486")]
    [InlineData("4111 1111 1111 1111", "************1111")]
    [InlineData("1234", "1234")]
    public void Mask_KeepsLastFour(string number, string expected) => Assert.Equal(expected, CardNumber.Mask(number));

    [Theory]
    [InlineData("4539578763621486", "visa")]
    [InlineData("5500005555555559", "mastercard")]
    [InlineData("378282246310005", "amex")]
    [InlineData("9999999999999999", "unknown")]
    public void Brand_ComesFromTheFirstDigit(string number, string brand)
        => Assert.Equal(brand, CardNumber.Brand(number));
}
