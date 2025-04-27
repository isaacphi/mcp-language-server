"""A clean Python module without any errors or warnings."""

from typing import Optional, Tuple


def SameName():
    pass


def clean_function(param: str) -> str:
    """A clean function without errors.

    Args:
        param: The input parameter

    Returns:
        The processed result
    """
    return f"Processed: {param}"


class CleanClass:
    """A clean class without errors."""

    def __init__(self, name: str):
        """Initialize a CleanClass instance.

        Args:
            name: The name of this instance
        """
        self.name = name

    def get_name(self) -> str:
        """Get the name of this instance.

        Returns:
            The name of this instance
        """
        return self.name

    @staticmethod
    def utility_method(items: list[int]) -> int:
        """Calculate the sum of a list of integers.

        Args:
            items: A list of integers

        Returns:
            The sum of the integers
        """
        return sum(items)


# Clean constants and variables
CLEAN_CONSTANT: str = "This is a clean constant"
clean_variable: list[int] = [10, 20, 30, 40, 50]
