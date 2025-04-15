package procweb

// ## objectives
// - test different lua programs
// - test different valid inputs
// - test a variety of invalid inputs
// - test a variety of failure conditions
//		- socket errors
//		- lua process errors

// ## what constitutes a correct result?
// - client receives the expected json messages
// - all goroutines shut down when they should
// - there should be no case where any of the goroutines crash (it's ok if the exec process crashes)

// ## what units need to be tested?
// jsonFromMsg
// ScanProcConnection
// SendProcConnection
// inScanner
// outScanner

// We also need to test the whole subsystem (aka NewInstance)
