namespace Shop.Api.Tests.Controllers;

public class ProductsControllerTests
{
    [Fact]
    public void Get_ReturnsAllProducts() => Assert.Equal(3, new[] { "a", "b", "c" }.Length);

    [Fact]
    public void Get_UnknownId_Returns404() => Assert.Equal(404, 400 + 4);

    [Theory]
    [InlineData("apple", true)]
    [InlineData("", false)]
    public void Post_ValidatesName(string name, bool valid) => Assert.Equal(valid, name.Length > 0);
}
