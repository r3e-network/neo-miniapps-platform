//! SGX Test Application
//!
//! This is a simple test application to verify the SGX enclave works correctly.
//! In production, the enclave is accessed via Go CGO through the bridge library.

fn main() {
    println!("SGX Test Application");
    println!("====================");
    println!();
    println!("This application is a placeholder for testing the SGX enclave.");
    println!("The actual integration is done via:");
    println!("  1. Go application (service_layer)");
    println!("  2. CGO bridge (sgx_hardware.go)");
    println!("  3. Rust bridge library (libsgx_bridge.so)");
    println!("  4. SGX enclave (tee_enclave.signed.so)");
    println!();
    println!("To build and test:");
    println!("  cd system/tee/sgx_enclave");
    println!("  make              # Build in simulation mode");
    println!("  make SGX_MODE=HW  # Build in hardware mode");
    println!("  make test         # Run tests");
    println!();
    println!("To use from Go:");
    println!("  go build -tags sgx ./...");
}
