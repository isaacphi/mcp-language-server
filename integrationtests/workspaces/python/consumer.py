"""Consumer module that uses the helper module."""

from typing import List, Dict
from helper import (
    helper_function, 
    get_items, 
    SharedClass, 
    SharedInterface, 
    SHARED_CONSTANT,
    Color
)


class MyImplementation(SharedInterface):
    """An implementation of the SharedInterface."""
    
    def process(self, data: List[str]) -> Dict[str, int]:
        """Process the given data by counting occurrences.
        
        Args:
            data: List of strings to process
            
        Returns:
            Dictionary with counts of each item
        """
        result = {}
        for item in data:
            if item in result:
                result[item] += 1
            else:
                result[item] = 1
        return result


def consumer_function() -> None:
    """Function that consumes the helper functions."""
    # Use the helper function
    message = helper_function("World")
    print(message)
    
    # Get and process items from the helper
    items = get_items()
    for item in items:
        print(f"Processing {item}")
    
    # Use the shared class
    shared = SharedClass[str]("consumer", SHARED_CONSTANT)
    print(f"Using shared class: {shared.get_name()} - {shared.get_value()}")
    
    # Use our implementation of the shared interface
    impl = MyImplementation()
    result = impl.process(items)
    print(f"Processed items: {result}")
    
    # Use the enum
    color = Color.RED
    print(f"Selected color: {color}")


def process_data() -> None:
    """Process some sample data."""
    data = get_items()
    print(f"Found {len(data)} items")
    
    # Sort and display the data
    sorted_data = sorted(data)
    print(f"Sorted data: {sorted_data}")
    
    # Count the items
    counts = {}
    for item in data:
        if item in counts:
            counts[item] += 1
        else:
            counts[item] = 1
    
    print(f"Item counts: {counts}")
    

if __name__ == "__main__":
    consumer_function()
    process_data()