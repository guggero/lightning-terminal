import BaseApi from './base';
import GrpcClient from './grpc';

/** the names and argument types for the subscription events */
// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface LlmEvents {}

/**
 * An API wrapper to communicate with the LLM node via GRPC
 */
class LlmApi extends BaseApi<LlmEvents> {
  private _grpc: GrpcClient;

  constructor(grpc: GrpcClient) {
    super();
    this._grpc = grpc;
  }
}

export default LlmApi;
