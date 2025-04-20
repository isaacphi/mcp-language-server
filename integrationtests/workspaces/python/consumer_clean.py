"""Consumer module that uses the helper module."""

from helper import helper_function, get_items


def consumer_function() -> None:
    """Function that consumes the helper functions."""
    # Use the helper function
    message = helper_function("World")
    print(message)

    # Get and process items from the helper
    items = get_items()
    for item in items:
        print(f"Processing {item}")


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
