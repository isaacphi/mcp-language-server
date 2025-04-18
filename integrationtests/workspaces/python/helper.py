"""Helper module that provides utility functions."""

from typing import List, Dict


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