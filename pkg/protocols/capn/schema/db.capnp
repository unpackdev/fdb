@0xd52c062bf4b48979;  # Unique file ID

using Go = import "/go.capnp";
$Go.package("schema");
$Go.import("protocols/capn/schema");

struct Request {
  union {
    get :group {
      key @0 :Data;
    }
    set :group {
      key @1 :Data;
      value @2 :Data;
    }
    delete :group {
      key @3 :Data;
    }
    exists :group {
      key @4 :Data;
    }
  }
}

struct Response {
  union {
    get :group {
      value @0 :Data;
      exists @1 :Bool;
    }
    set :group {
      success @2 :Bool;
    }
    delete :group {
      success @3 :Bool;
    }
    exists :group {
      exists @4 :Bool;
    }
    error :group {
      message @5 :Text;
    }
  }
}
