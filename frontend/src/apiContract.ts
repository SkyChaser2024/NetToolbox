import type * as GeneratedAPI from '../wailsjs/go/desktop/App'
import type { BackendAPI } from './api'

// Wails model classes contain conversion methods which are absent from JSON.
// Widen domain string unions only for this check; UI consumers keep strict types.
type WireData<T> = T extends string
  ? string
  : T extends readonly unknown[]
    ? { [K in keyof T]: WireData<T[K]> }
    : T extends object
      ? { [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]: WireData<T[K]> }
      : T

type WireAPI<T> = {
  [K in keyof T]: T[K] extends (...args: infer Args) => Promise<infer Result>
    ? { arguments: WireData<Args>; result: WireData<Result> }
    : never
}

type Equal<Left, Right> =
  (<T>() => T extends Left ? 1 : 2) extends <T>() => T extends Right ? 1 : 2 ? true : false
type Assert<Match extends true> = Match

// Adding/removing a method or changing a DTO field must update the facade too.
// This is compiled by tsc and has no runtime import of generated bindings.
export type BackendAPIContract = Assert<Equal<WireAPI<BackendAPI>, WireAPI<typeof GeneratedAPI>>>
