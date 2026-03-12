// Placeholder file for another_consumer.cpp
#include <iostream>

void anotherConsume() {
  std::cout << "Another consume function" << std::endl;
#if 0
  // This block is intentionally inactive for testing clangd inactiveRegions.
  std::cout << "Inactive code path" << std::endl;
#endif
}