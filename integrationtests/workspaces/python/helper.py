"""Helper module that provides utility functions."""

from typing import List, Dict, TypeVar, Generic
from enum import Enum


# Shared constant used across files
SHARED_CONSTANT = "SHARED_VALUE"


# Enum-like class that will be referenced across files
class Color(Enum):
    """Color enumeration used across files."""
    RED = "red"
    GREEN = "green"
    BLUE = "blue"


# Generic type variable for SharedClass
T = TypeVar('T')


# Shared class that will be referenced across files
class SharedClass(Generic[T]):
    """A shared class that is used across multiple files."""
    
    def __init__(self, name: str, value: T):
        """Initialize with a name and value.
        
        Args:
            name: The name of this instance
            value: The value to store
        """
        self.name = name
        self.value = value
    
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


# Interface-like class (abstract base class in Python)
class SharedInterface:
    """An interface-like class that defines a contract."""
    
    def process(self, data: List[str]) -> Dict[str, int]:
        """Process the given data.
        
        Args:
            data: List of strings to process
            
        Returns:
            Dictionary with processing results
        """
        raise NotImplementedError("Implementations must override process")


def helper_function(name: str) -> str:
    """A helper function that formats a greeting message.
    
    Args:
        name: The name to greet
        
    Returns:
        A formatted greeting message
    """
    return f"Hello, {name}!"


def get_items() -> List[str]:
    """Get a list of sample items.
    
    Returns:
        A list of sample strings
    """
    return ["apple", "banana", "orange", "grape"]