"""A Python module with deliberate errors for testing diagnostics."""

from typing import Any


def function_with_unreachable_code(value: int) -> str:
    """A function with unreachable code.

    Args:
        value: An integer value

    Returns:
        A string result
    """
    if value > 0:
        return "Positive"
    elif value < 0:
        return "Negative"
    else:
        return "Zero"
        # This is unreachable code
        print("This will never be executed")


def function_with_type_error() -> str:
    """A function with a type error.

    Returns:
        Should return a string but actually returns an int
    """
    return 42  # Type error: Incompatible return value type (got "int", expected "str")


class ErrorClass:
    """A class with errors."""

    def __init__(self, value: dict[str, Any]):
        """Initialize with errors.

        Args:
            value: A dictionary
        """
        self.value = value

    def method_with_undefined_variable(self) -> None:
        """A method that uses an undefined variable."""
        print(undefined_variable)  # Error: undefined_variable is not defined


# Variable with incompatible type annotation
wrong_type: str = 123  # Type error: Incompatible types in assignment
