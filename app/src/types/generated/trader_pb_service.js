// package: poolrpc
// file: trader.proto

var trader_pb = require("./trader_pb");
var auctioneer_pb = require("./auctioneer_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var Trader = (function () {
  function Trader() {}
  Trader.serviceName = "poolrpc.Trader";
  return Trader;
}());

Trader.QuoteAccount = {
  methodName: "QuoteAccount",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.QuoteAccountRequest,
  responseType: trader_pb.QuoteAccountResponse
};

Trader.InitAccount = {
  methodName: "InitAccount",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.InitAccountRequest,
  responseType: trader_pb.Account
};

Trader.ListAccounts = {
  methodName: "ListAccounts",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.ListAccountsRequest,
  responseType: trader_pb.ListAccountsResponse
};

Trader.CloseAccount = {
  methodName: "CloseAccount",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.CloseAccountRequest,
  responseType: trader_pb.CloseAccountResponse
};

Trader.WithdrawAccount = {
  methodName: "WithdrawAccount",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.WithdrawAccountRequest,
  responseType: trader_pb.WithdrawAccountResponse
};

Trader.DepositAccount = {
  methodName: "DepositAccount",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.DepositAccountRequest,
  responseType: trader_pb.DepositAccountResponse
};

Trader.BumpAccountFee = {
  methodName: "BumpAccountFee",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.BumpAccountFeeRequest,
  responseType: trader_pb.BumpAccountFeeResponse
};

Trader.RecoverAccounts = {
  methodName: "RecoverAccounts",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.RecoverAccountsRequest,
  responseType: trader_pb.RecoverAccountsResponse
};

Trader.SubmitOrder = {
  methodName: "SubmitOrder",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.SubmitOrderRequest,
  responseType: trader_pb.SubmitOrderResponse
};

Trader.ListOrders = {
  methodName: "ListOrders",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.ListOrdersRequest,
  responseType: trader_pb.ListOrdersResponse
};

Trader.CancelOrder = {
  methodName: "CancelOrder",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.CancelOrderRequest,
  responseType: trader_pb.CancelOrderResponse
};

Trader.AuctionFee = {
  methodName: "AuctionFee",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.AuctionFeeRequest,
  responseType: trader_pb.AuctionFeeResponse
};

Trader.LeaseDurations = {
  methodName: "LeaseDurations",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.LeaseDurationRequest,
  responseType: trader_pb.LeaseDurationResponse
};

Trader.NextBatchInfo = {
  methodName: "NextBatchInfo",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.NextBatchInfoRequest,
  responseType: trader_pb.NextBatchInfoResponse
};

Trader.BatchSnapshot = {
  methodName: "BatchSnapshot",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: auctioneer_pb.BatchSnapshotRequest,
  responseType: auctioneer_pb.BatchSnapshotResponse
};

Trader.GetLsatTokens = {
  methodName: "GetLsatTokens",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.TokensRequest,
  responseType: trader_pb.TokensResponse
};

Trader.Leases = {
  methodName: "Leases",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.LeasesRequest,
  responseType: trader_pb.LeasesResponse
};

Trader.NodeRatings = {
  methodName: "NodeRatings",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: trader_pb.NodeRatingRequest,
  responseType: trader_pb.NodeRatingResponse
};

Trader.BatchSnapshots = {
  methodName: "BatchSnapshots",
  service: Trader,
  requestStream: false,
  responseStream: false,
  requestType: auctioneer_pb.BatchSnapshotsRequest,
  responseType: auctioneer_pb.BatchSnapshotsResponse
};

exports.Trader = Trader;

function TraderClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

TraderClient.prototype.quoteAccount = function quoteAccount(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.QuoteAccount, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.initAccount = function initAccount(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.InitAccount, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.listAccounts = function listAccounts(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.ListAccounts, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.closeAccount = function closeAccount(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.CloseAccount, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.withdrawAccount = function withdrawAccount(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.WithdrawAccount, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.depositAccount = function depositAccount(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.DepositAccount, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.bumpAccountFee = function bumpAccountFee(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.BumpAccountFee, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.recoverAccounts = function recoverAccounts(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.RecoverAccounts, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.submitOrder = function submitOrder(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.SubmitOrder, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.listOrders = function listOrders(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.ListOrders, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.cancelOrder = function cancelOrder(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.CancelOrder, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.auctionFee = function auctionFee(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.AuctionFee, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.leaseDurations = function leaseDurations(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.LeaseDurations, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.nextBatchInfo = function nextBatchInfo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.NextBatchInfo, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.batchSnapshot = function batchSnapshot(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.BatchSnapshot, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.getLsatTokens = function getLsatTokens(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.GetLsatTokens, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.leases = function leases(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.Leases, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.nodeRatings = function nodeRatings(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.NodeRatings, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

TraderClient.prototype.batchSnapshots = function batchSnapshots(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(Trader.BatchSnapshots, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

exports.TraderClient = TraderClient;

