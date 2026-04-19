export class LatticeError extends Error {}

export function parseDocument(content: string): unknown;
export function parse(content: string): unknown;
export function parseBuffer(content: Uint8Array | Buffer): unknown;
export function stringify(value: unknown): string;
export function toBuffer(value: unknown): Buffer;
