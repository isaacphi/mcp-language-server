"""Module that uses helpers and shared components."""

from typing import Any, Dict, List

from helper import (
    SHARED_CONSTANT,
    PI,
    SharedClass,
    SharedInterface,
    helper_function,
    calculate_area,
    DataDict,
    Color,
)


class Consumer(SharedInterface):
    """A class that implements the SharedInterface."""
    
    def __init__(self):
        """Initialize the Consumer."""
        self.data: List[str] = []
    
    def process(self, data: str) -> str:
        """Process the input data by converting to uppercase.
        
        Args:
            data: The input data to process
            
        Returns:
            The processed data
        """
        result = helper_function(data)  # Using the helper function
        self.data.append(result)
        return result
    
    def validate(self, value: Any) -> bool:
        """Validate that the value is a non-empty string.
        
        Args:
            value: The value to validate
            
        Returns:
            True if valid, False otherwise
        """
        return isinstance(value, str) and len(value) > 0


def consumer_function() -> None:
    """Function that uses various shared components."""
    # Use shared constants
    print(f"Shared constant: {SHARED_CONSTANT}")
    
    # Use shared class
    shared = SharedClass[int]("example", 42)
    name = shared.get_name()
    value = shared.get_value()
    print(f"Shared class name: {name}, value: {value}")
    
    # Set new value
    shared.set_value(100)
    
    # Use shared interface
    consumer = Consumer()
    processed = consumer.process("test data")
    print(f"Processed: {processed}")
    
    # Use helper function directly
    result = helper_function("direct call")
    print(f"Helper result: {result}")
    
    # Use another helper function with PI constant
    area = calculate_area(2.0)
    print(f"Circle area: {area}")
    
    # Use type alias
    data: DataDict = {
        "name": "test",
        "value": 42,
        "enabled": True
    }
    print(f"Data: {data}")
    
    # Use enum-like class
    color = Color.BLUE
    print(f"Color: {color}")


if __name__ == "__main__":
    consumer_function()