// Placeholder file for consumer.cpp
#include <iostream>
#include "helper.hpp"

void consume() { std::cout << "Consume function" << std::endl; }

class TestClass {
 public:
  /**
   * @brief A method that takes an integer parameter.
   *
   * @param param The integer parameter to be processed.
   */
  void method(int param) { helperFunction(); }
};