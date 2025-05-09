#include <iostream>
#include "helper.hpp"

// FooBar is a simple function for testing
void foo_bar() {
  std::cout << "Hello, World!" << std::endl;
  return;
}

int main() {
  helperFunction();
  return 0;

  foo_bar();

  // Intentional error: unreachable code
  std::cout << "This is unreachable" << std::endl;
}