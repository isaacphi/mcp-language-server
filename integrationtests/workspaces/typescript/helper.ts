// Helper functions and types that are used across files

// SharedFunction with references across files
export function SharedFunction(): string {
  return "helper function";
}

// SharedInterface with methods
export interface SharedInterface {
  getName(): string;
  getValue(): number;
}

// SharedClass implementing the interface
export class SharedClass implements SharedInterface {
  private name: string;

  constructor(name: string) {
    this.name = name;
  }
  
  getName(): string {
    return this.name;
  }
  
  getValue(): number {
    return 42;
  }
  
  helperMethod(): void {
    console.log("Helper method called");
  }
}

// SharedType referenced across files
export type SharedType = string | number;

// SharedConstant referenced across files
export const SharedConstant = "SHARED_VALUE";

// SharedEnum referenced across files
export enum SharedEnum {
  ONE = "one",
  TWO = "two",
  THREE = "three"
}