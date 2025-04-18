// Another consumer file that uses elements from the helper file
import { 
  SharedFunction, 
  SharedInterface, 
  SharedClass, 
  SharedType, 
  SharedConstant, 
  SharedEnum 
} from './helper';

// AnotherConsumerFunction uses SharedFunction in a different way
export function AnotherConsumerFunction(): void {
  const result = SharedFunction();
  console.log(`Result from shared function: ${result}`);
  
  // Using SharedClass differently
  const instance = new SharedClass("another instance");
  
  // Using SharedInterface
  const iface: SharedInterface = {
    getName: () => "custom implementation",
    getValue: () => 100
  };
  
  // Using SharedType
  const mixedArray: SharedType[] = ["string", 42, "another"];
  
  // Using SharedConstant
  const prefixed = `PREFIX_${SharedConstant}`;
  
  // Using SharedEnum
  const enumValues = [SharedEnum.ONE, SharedEnum.TWO, SharedEnum.THREE];
  
  console.log(instance, iface, mixedArray, prefixed, enumValues);
}