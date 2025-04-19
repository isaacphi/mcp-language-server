// Consumer module for testing references
use crate::helper::helper_function;
use crate::types::{
    SharedInterface, SharedStruct, SharedType, SHARED_CONSTANT,
};

pub fn consumer_function() {
    // Use the helper function
    let result = helper_function();
    println!("Helper result: {}", result);

    // Use shared struct
    let s = SharedStruct::new("test");
    println!("Struct method: {}", s.method());

    // Use shared interface
    let iface: &dyn SharedInterface = &s;
    println!("Interface method: {}", iface.get_name());

    // Use shared constant
    println!("Constant: {}", SHARED_CONSTANT);

    // Use shared type
    let t: SharedType = String::from("test");
    println!("Type: {}", t);
}
