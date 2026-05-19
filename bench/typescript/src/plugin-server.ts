import { serve } from '../../../runtimes/typescript/dist';

const CONTRACT_HASH = 'sha256:pluginart-bench';

serve((payload: Buffer) => payload, { contractHash: CONTRACT_HASH });
