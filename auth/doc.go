// Package auth implements AWS credential process with caching.
// Cache files are saved on disk and encrypted via AES-GCM with the encryption key stored in the operating system's "native" credentials store.
// For example, Keychain is used on macOS.
package auth
