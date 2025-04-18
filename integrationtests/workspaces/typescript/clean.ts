// This file has no errors or diagnostics
export function cleanFunction(): string {
  return "This is a clean function";
}

export class CleanClass {
  private value: string;

  constructor(initialValue: string) {
    this.value = initialValue;
  }

  getValue(): string {
    return this.value;
  }
}

function runClean(): void {
  const instance = new CleanClass("test");
  console.log(instance.getValue());
  console.log(cleanFunction());
}

export default runClean;