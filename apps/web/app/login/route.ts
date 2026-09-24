// /login is an alias of the sign-in page, not a page of its own. Forward with a 307
// (an alias, not a moved page: a cached 308 would loop if sign-in ever moved here)
// and carry no query string, so a returnUrl on the alias is never passed on.
//
// This is a route handler on purpose: a `redirects()` entry in next.config.js was
// measured to pass the query through, and a page calling `redirect()` answered 200
// with no Location header.
export function GET() {
  return new Response(null, { status: 307, headers: { Location: "/auth/login" } });
}
