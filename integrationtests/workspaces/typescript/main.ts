// TestFunction is a simple function for testing
export function TestFunction(): string {
  return "Hello, World!";
  console.log("Unreachable code"); // This is unreachable code
}

// TestInterface definition
export interface TestInterface {
  method(): void;
  property: string;
}

// TestClass with method
export class TestClass implements TestInterface {
  property: string;
  
  constructor() {
    this.property = "test";
  }
  
  method(): void {
    console.log("Method called");
  }
}

// TestType definition
export type TestType = string | number;

// TestVariable definition
export const TestVariable: string = "Test";

// TestConstant definition
export const TestConstant = 42;

// Main function
function main() {
  console.log(TestFunction());
  const instance = new TestClass();
  instance.method();
}

main();