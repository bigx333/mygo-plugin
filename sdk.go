package plugin

// HostTypeScriptVersion is the version of the host global that HostTypeScript
// describes. The host bumps it when the global changes.
const HostTypeScriptVersion = 2

// HostTypeScript declares the host global that a plugin's TypeScript sees.
// spec/host.d.ts holds the same bytes for editors; sdk_test.go keeps the two
// from drifting.
const HostTypeScript = `// host.d.ts — the host global this host installs, generated for mygo-plugin
// v2. It lists exactly what a plugin can reach; host.spawn is not available,
// because this host starts no programs for plugins.

declare const host: {
	/** The host's operating system: "darwin", "linux", "windows". */
	readonly platform: string;
	/** The host's architecture: "arm64", "amd64". */
	readonly arch: string;
	/** The plugin's own folder. */
	readonly dir: string;
	/** The folder the plugin keeps its data in, one per plugin. */
	readonly dataDir: string;

	/** The listener's language, a BCP 47 tag such as "zh-CN". */
	language(): string;
	/** The value the listener gave one setting of this plugin; "" if unset. */
	setting(key: string): string;
	/** Whether the host offers a capability the plugin asked for, such as "network". */
	capability(name: string): boolean;
	/** Opens a web page. Only http and https are accepted. */
	openURL(url: string): void;
	/** Shows what the plugin wants shown while the listener signs in. */
	signInPrompt(prompt: { qr?: string; text?: string }): void;

	/** A JSON key-value store, kept in storage.json in dataDir. */
	storage: {
		get(key: string): unknown;
		set(key: string, value: unknown): void;
		remove(key: string): void;
	};

	crypto: {
		/** The text's MD5, in lowercase hex. */
		md5(text: string): string;
		/** AES-ECB with PKCS#7 padding; the output is hex. */
		aesEcbEncrypt(key: string, text: string): string;
	};
};
`
