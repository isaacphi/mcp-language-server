"""Another module that uses helpers and shared components."""

from helper import (
    SHARED_CONSTANT,
    SharedClass,
    helper_function,
    Color,
)


class AnotherImplementation:
    """A class that uses shared components but doesn't implement interfaces."""
    
    def __init__(self):
        """Initialize the implementation."""
        self.shared = SharedClass[str]("another", SHARED_CONSTANT)
    
    def do_something(self) -> str:
        """Do something with the shared components.
        
        Returns:
            The processed result
        """
        # Get the value from shared class
        value = self.shared.get_value()
        
        # Process it using the helper function
        return helper_function(value)


def another_consumer_function() -> None:
    """Another function that uses various shared components."""
    # Use shared constants
    print(f"Using constant: {SHARED_CONSTANT}")
    
    # Use shared class with a different type parameter
    shared = SharedClass[float]("another example", 3.14)
    
    # Use methods from shared class
    name = shared.get_name()
    value = shared.get_value()
    print(f"Name: {name}, Value: {value}")
    
    # Use our own implementation
    impl = AnotherImplementation()
    result = impl.do_something()
    print(f"Implementation result: {result}")
    
    # Use helper function
    output = helper_function("another direct call")
    print(f"Helper output: {output}")
    
    # Use enum-like class with a different color
    color = Color.GREEN
    print(f"Selected color: {color}")


if __name__ == "__main__":
    another_consumer_function()