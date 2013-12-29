var hakobiyaModule = angular.module('hakobiya', []);

hakobiyaModule.factory('Hakobiya', function($rootScope) {
	var Hakobiya = {
		socket: null,
		sendQueue: [],
		jpCount: {},
		chanQueue: {},
		URL: null,

		connect: function(addr) {
			var self = this;

			this.URL = addr;
			this.socket = new WebSocket(addr);
			this.socket.onopen = function() {
				console.log("Hakobiya: connected.");

				angular.forEach(self.sendQueue, function(msg) {
				    self.send(msg);
				});
				self.sendQueue = [];
			};
			this.socket.onclose = function() {
				console.log("Hakobiya: disconnected.");
			};
			this.socket.onerror = function (error) {
			    console.log(error);
			};
			this.socket.onmessage = function(evt) {
				var data = angular.fromJson(evt.data);
				switch (data.x) {
					case 's': //set
						var id = data.c + "." + data.n;
						$rootScope.$broadcast(id, data.v);
						break;
					case 'j': //joined
						self.jpCount[data.c] = 1;
						// send any waiting msgs
						angular.forEach(self.chanQueue[data.c], function(msg) {
							self.send(msg);
						});
						self.chanQueue[data.c] = [];
						// broadcast join event
						var joinEvt = data.c + ":join";
						$rootScope.$broadcast(joinEvt, true); 
						break;
					case '!': //error
						console.log("error (" + data.w + "): " + data.m);
						break;
				}
			};
		},
		send: function(data) {
			if (this.socket && this.socket.readyState == 1) {
				this.socket.send(angular.toJson(data));
			} else {
				this.sendQueue.push(data);
			}
		},
		sendTo: function(channel, data) {
			if (this.joined(channel)) {
				this.send(data);
			} else {
				// enqueue a msg for when we join
				if (this.chanQueue[channel]) {
					this.chanQueue[channel].push(data);
				} else {
					this.chanQueue[channel] = [data];
				}
			}	
		},
		joined: function(channel) {
			return this.jpCount[channel] > 0;
		},
		join: function(channel) {
			if (!this.jpCount[channel]) {
				this.send({
					x: 'j',
					c: channel
				});
			} else {
				this.jpCount[channel] += 1;
			}
		},
		part: function(channel) {
			this.jpCount[channel] -= 1;
			if (this.jpCount[channel] < 1) {
				this.send({
					x: 'p',
					c: channel
				});
			}
		},
		get: function(channel, v) {
			this.sendTo(channel, {
				x: 'g',
				c: channel,
				n: v
			});
		},
		multiget: function(channel, vars) {
			this.sendTo(channel, {
				x: 'G',
				c: channel,
				n: vars
			});
		},
		set: function(channel, variable, value) {
			this.sendTo(channel, {
				x: 's',
				c: channel,
				n: variable,
				v: value
			});
		},
		multiset: function(channel, vars) {
			this.sendTo(channel, {
				x: 'S',
				c: channel,
				v: vars,
			});
		},
		bind: function($scope, chvar, binding) {
			// is our channel null? if so we have to do this later...
			var ch = $scope[chvar];
			if (!ch) {
				var self = this;
				var unreg = $scope.$watch(chvar, function(newVal, oldVal) {
					self._do_bind($scope, newVal, binding);
					// for now, we only do this once
					// TODO: changing channels
					unreg();
				});
			} else {
				this._do_bind($scope, ch, binding);
			}
		},
		// ----- here be dragons -----
		_do_bind: function($scope, chan, binding) {
			var self = this;

			this.join(chan);
			$scope.$on("$destroy", function() {
				self.part(chan);
			});

			// let the binding begin
			var requestableSigils = "&$%";
			var settableSigils = "%";
			var request = [];
			angular.forEach(binding, function(hvar, scopevar) {
				var sigil = hvar[0];
				var requestable = requestableSigils.indexOf(sigil) != -1;
				if (requestable) {
					var value = $scope[scopevar];
					var settable = settableSigils.indexOf(sigil) != -1;
					if (!value) {
						request.push(hvar);
					} else if (settable) {
						self.set(chan, hvar, value);
					}
				}

				var id = chan + "." + hvar;
				switch (sigil) {
					case '&': // magic var, one-way server -> client binding
					case '$': // system var, same
						$scope.$on(id, function(e, value) {
							$scope.$apply(function(scope) {
								scope[scopevar] = value;
							});
						});
						break;
					case '%': // uservars, two-way binding
						$scope.$on(id, function(e, value) {
							$scope.$apply(function(scope) {
								scope[scopevar] = value;
							});
						});
						$scope.$watch(scopevar, function(newVal, oldVal) {
							self.set(chan, hvar, newVal);
						});
						break;
					case '#': // broadcasts, one way growing array
						$scope.$on(id, function(e, value) {
							$scope.$apply(function(scope) {
								if (scope[scopevar]) {
									scope[scopevar].push(value);
								} else {
									scope[scopevar] = [value];
								}
							});
						});
						break;
					case '=': // wire, like a two-way broadcast. special array
						var wire = [];
						wire.id = id;
						wire.enabled = false;
						wire.enable = function() {
							var disableFn = $scope.$on(id, function(e, value) {
								$scope.$apply(function(scope) {
									scope[scopevar].push(value);
								});
							});
							wire.disable = function() {
								disableFn();
								wire.enabled = false;
							};
							wire.enabled = true;
						};
						wire.send = function(data) {
							self.set(chan, hvar, data);
						};
						wire.toString = function() {
							return this.id;
						};
						wire.enable();
						$scope[scopevar] = wire;
						break;
					default:
						console.log("ERROR: unknown sigil " + hvar);
						break;
				}
			});
			this.multiget(chan, request);	
		}
	};
	return Hakobiya;
});