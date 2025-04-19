// Another consumer module for testing references
use crate::helper::helper_function;
use crate::types::{
    SharedInterface, SharedStruct, SharedType, SHARED_CONSTANT,
};

pub fn another_consumer_function() {
    // Use the helper function
    let result = helper_function();
    println!("Helper result from another consumer: {}", result);

    // Use shared struct
    let s = SharedStruct::new("another test");
    println!("Struct in another consumer: {}", s.name);

    // Use shared interface
    let _iface: &dyn SharedInterface = &s;
    
    // Use shared constant
    println!("Constant in another consumer: {}", SHARED_CONSTANT);

    // Use shared type
    let _t: SharedType = String::from("another test");
}