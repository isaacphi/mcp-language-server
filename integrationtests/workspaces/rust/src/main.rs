// Main module for testing Rust integration
mod types;
mod helper;
mod consumer;
mod another_consumer;
mod clean;

// FooBar is a simple function for testing
fn foo_bar() -> String {
    String::from("Hello, World!")
    println!("Unreachable code"); // This is unreachable code
}

fn main() {
    println!("{}", foo_bar());
}