// Consumer file that uses elements from the helper file
import { 
  SharedFunction, 
  SharedInterface, 
  SharedClass, 
  SharedType, 
  SharedConstant, 
  SharedEnum 
} from './helper';

// ConsumerFunction uses SharedFunction
export function ConsumerFunction(): void {
  console.log("Consumer calling:", SharedFunction());
  
  // Using SharedClass
  const instance = new SharedClass("test instance");
  console.log(instance.getName());
  instance.helperMethod();
  
  // Using SharedInterface
  const iface: SharedInterface = instance;
  console.log(iface.getName());
  console.log(iface.getValue());
  
  // Using SharedType
  const value: SharedType = "string value";
  const numValue: SharedType = 42;
  console.log(value, numValue);
  
  // Using SharedConstant
  console.log(SharedConstant);
  
  // Using SharedEnum
  console.log(SharedEnum.ONE);
}

// Call the function
ConsumerFunction();