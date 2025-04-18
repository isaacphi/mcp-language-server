"""Module containing test definitions for Python LSP integration tests."""

from typing import List, Dict, Optional, Union


def test_function(name: str) -> str:
    """A simple test function that returns a greeting message.

    Args:
        name: The name to greet

    Returns:
        A greeting message
    """
    return f"Hello, {name}!"


class TestClass:
    """A test class with methods and attributes."""

    class_variable: str = "class variable"

    def __init__(self, value: int = 0):
        """Initialize the TestClass.

        Args:
            value: The initial value
        """
        self.value: int = value

    def test_method(self, increment: int) -> int:
        """Increment the value by the given amount.

        Args:
            increment: The amount to increment by

        Returns:
            The new value
        """
        self.value += increment
        return self.value

    @staticmethod
    def static_method(items: List[str]) -> Dict[str, int]:
        """Convert a list of items to a dictionary with item counts.

        Args:
            items: A list of strings

        Returns:
            A dictionary mapping items to their counts
        """
        result: Dict[str, int] = {}
        for item in items:
            if item in result:
                result[item] += 1
            else:
                result[item] = 1
        return result


class BaseClass:
    """A base class for inheritance testing."""

    def base_method(self) -> None:
        """A method defined in the base class."""
        pass


class DerivedClass(BaseClass):
    """A class that inherits from BaseClass."""

    def derived_method(self) -> None:
        """A method defined in the derived class."""
        pass


# Constants
TEST_CONSTANT: str = "test constant"
PI: float = 3.14159

# Variables
test_variable: List[int] = [1, 2, 3, 4, 5]
optional_var: Optional[str] = None
union_var: Union[int, str] = "test"


def main() -> None:
    """Main function that demonstrates usage of the defined symbols."""
    result = test_function("World")
    print(result)

    obj = TestClass(10)
    new_value = obj.test_method(5)
    print(f"New value: {new_value}")

    counts = TestClass.static_method(["apple", "banana", "apple", "orange"])
    print(f"Counts: {counts}")

    print(f"Constants - TEST_CONSTANT: {TEST_CONSTANT}, PI: {PI}")
    print(f"Variables - test_variable: {test_variable}")


if __name__ == "__main__":
    main()

