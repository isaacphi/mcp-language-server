"""Module containing shared helper functions and classes."""

from typing import Any, Dict, List, Optional, Union, TypeVar, Generic

# Constants
SHARED_CONSTANT: str = "shared constant value"
PI: float = 3.14159

# Types
T = TypeVar('T')


class SharedClass(Generic[T]):
    """A shared class that can be used by other modules."""
    
    def __init__(self, name: str, value: T):
        """Initialize the SharedClass.
        
        Args:
            name: The name of the instance
            value: The value to store
        """
        self.name: str = name
        self.value: T = value
    
    def get_name(self) -> str:
        """Get the name of this instance.
        
        Returns:
            The name string
        """
        return self.name
    
    def get_value(self) -> T:
        """Get the stored value.
        
        Returns:
            The stored value
        """
        return self.value
    
    def set_value(self, value: T) -> None:
        """Set a new value.
        
        Args:
            value: The new value to store
        """
        self.value = value


class SharedInterface:
    """An interface-like class that defines methods to be implemented."""
    
    def process(self, data: str) -> str:
        """Process the input data.
        
        Args:
            data: The input data to process
            
        Returns:
            The processed data
        """
        raise NotImplementedError("Subclasses must implement this method")
    
    def validate(self, value: Any) -> bool:
        """Validate the given value.
        
        Args:
            value: The value to validate
            
        Returns:
            True if valid, False otherwise
        """
        raise NotImplementedError("Subclasses must implement this method")


# Shared functions
def helper_function(text: str) -> str:
    """A helper function that can be used by other modules.
    
    Args:
        text: The input text
        
    Returns:
        The processed text
    """
    return text.upper()


def calculate_area(radius: float) -> float:
    """Calculate the area of a circle.
    
    Args:
        radius: The radius of the circle
        
    Returns:
        The area of the circle
    """
    return PI * radius * radius


# Type alias
DataDict = Dict[str, Union[str, int, float, bool, None]]


# Enum-like structure (Python 3.8 compatible)
class Color:
    """A simple enum-like class for colors."""
    RED = "red"
    GREEN = "green"
    BLUE = "blue"