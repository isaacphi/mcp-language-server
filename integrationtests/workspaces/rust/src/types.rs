// Types for testing

// A simple constant
pub const TEST_CONSTANT: &str = "test constant value";

// A simple variable
pub static TEST_VARIABLE: &str = "test variable value";

// A simple type alias
pub type TestType = String;

// A struct for testing
pub struct TestStruct {
    pub name: String,
    pub value: i32,
}

// Implementation for TestStruct
impl TestStruct {
    pub fn new(name: &str, value: i32) -> Self {
        TestStruct {
            name: String::from(name),
            value,
        }
    }

    pub fn method(&self) -> String {
        format!("{}: {}", self.name, self.value)
    }
}

// An interface (trait) for testing
pub trait TestInterface {
    fn get_name(&self) -> String;
    fn get_value(&self) -> i32;
}

// Implementation of TestInterface for TestStruct
impl TestInterface for TestStruct {
    fn get_name(&self) -> String {
        self.name.clone()
    }

    fn get_value(&self) -> i32 {
        self.value
    }
}

// Shared types for reference testing
pub struct SharedStruct {
    pub name: String,
}

impl SharedStruct {
    pub fn new(name: &str) -> Self {
        SharedStruct {
            name: String::from(name),
        }
    }

    pub fn method(&self) -> String {
        format!("SharedStruct: {}", self.name)
    }
}

pub trait SharedInterface {
    fn get_name(&self) -> String;
}

impl SharedInterface for SharedStruct {
    fn get_name(&self) -> String {
        self.name.clone()
    }
}

pub type SharedType = String;

pub const SHARED_CONSTANT: &str = "shared constant value";

// A simple function for testing
pub fn test_function() -> String {
    String::from("test function")
}
